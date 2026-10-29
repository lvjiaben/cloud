// Shared by offline migration and the already migrated public pages.
export function navigationToolsTemplate(){return `<div class="horizontal-menu-right-container">
 <div class="input-inner-wrapper">
  <label class="input-label"><input class="search-input" placeholder="云服务器" aria-label="打开搜索" aria-expanded="false" aria-controls="lina-navigation-search"><span class="input-suffix" aria-hidden="true"><i class="search-icon develop sk-sousuo1"></i></span></label>
  <div class="nav-search" id="lina-navigation-search" hidden>
   <div class="nav-search-body">
    <button type="button" class="nav-search-close" aria-label="关闭搜索">×</button>
    <ul class="nav-search-tabs" aria-label="搜索范围">
     <li class="nav-search-tab active"><button type="button" data-search-target="all" aria-pressed="true">全部产品</button></li>
     <li class="nav-search-tab"><button type="button" data-search-target="news" aria-pressed="false">新闻资讯</button></li>
     <li class="nav-search-tab"><button type="button" data-search-target="docs" aria-pressed="false">帮助文档</button></li>
    </ul>
    <form action="/products.html" class="lina-nav-search nav-search-input-wrapper"><input class="nav-search-input" name="namePrefix" placeholder="云服务器" aria-label="搜索商品"><button class="nav-search-input-suffix" type="submit" aria-label="搜索"><i class="search-icon develop sk-sousuo1" aria-hidden="true"></i></button></form>
   </div>
  </div>
 </div>
 <div class="navbar-public"><a href="/help.html">文档中心</a></div><div class="delimiter"></div>
 <div class="navbar-public"><a data-customer="/account">控制台</a></div><div class="delimiter"></div>
 <div class="navbar-Register"><a data-customer="/auth/login">登录</a></div>
 <a class="navbar-Login" data-customer="/auth/register">免费注册</a>
</div>`;}
